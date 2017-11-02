package protocol

import (
	"time"

	"github.com/dedis/student_17_bftcosi/cosi"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/log"
)

// Cosi holds the different channels used to receive the different protocol messages.
// It also defines a channel that will receive the final signature. Only the
// root-node will write to this channel.
type Cosi struct {
	*onet.TreeNodeInstance
	List                   []abstract.Point
	MinSubtreeSize         int // can be one more //TODO: see if still useful
	subleaderNotResponding chan bool
	FinalSignature         chan []byte
	ChannelAnnouncement    chan StructAnnouncement
	ChannelCommitment      chan []StructCommitment
	ChannelChallenge       chan StructChallenge
	ChannelResponse        chan []StructResponse
}

// Start is done only by root and sends the announcement message to all children
func (p *Cosi) Start() error {
	log.Lvl3("Starting Cosi")
	proposal := []byte{0xFF}
	announcement := StructAnnouncement{p.TreeNode(),
		Announcement{proposal}}
	p.ChannelAnnouncement <- announcement
	return nil
}

//Dispatch() is the main method of the protocol, handling the messages in order
func (p *Cosi) Dispatch() error {
	defer p.Done() //TODO: see if should stop node or be ready for another proposal

	// ----- Announcement -----
	announcement := <-p.ChannelAnnouncement
	log.Lvl3(p.ServerIdentity().Address, "received announcement")
	err := p.SendToChildren(&announcement.Announcement)
	if err != nil {
		return err
	}

	// ----- Commitment -----
	if p.IsLeaf() {
		p.ChannelCommitment <- make([]StructCommitment, 0)
	}
	commitments := make([]StructCommitment, 0)
	select {
	case commitments = <-p.ChannelCommitment:
	case <-time.After(Timeout):
		if p.IsRoot() {
			p.subleaderNotResponding <- true
			return nil
		}
	}
	log.Lvl3(p.ServerIdentity().Address, "received commitment")
	secret, commitment, mask, err := p.generateCommitment(commitments)
	if err != nil {
		return err
	}
	err = p.SendToParent(&Commitment{commitment, mask.Mask()})
	if err != nil {
		return err
	}

	// ----- Challenge -----
	if p.IsRoot() {
		cosiChallenge, err := cosi.Challenge(p.Suite(), commitment,
			p.Root().PublicAggregateSubTree, announcement.Proposal)
		if err != nil {
			return err
		}
		p.ChannelChallenge <- StructChallenge{p.TreeNode(), Challenge{cosiChallenge}}

	}
	challenge := <-p.ChannelChallenge
	log.Lvl3(p.ServerIdentity().Address, "received challenge")
	err = p.SendToChildren(&challenge.Challenge)
	if err != nil {
		return err
	}

	// ----- Response -----
	if p.IsLeaf() {
		p.ChannelResponse <- make([]StructResponse, 0)
	}
	responses := <-p.ChannelResponse
	log.Lvl3(p.ServerIdentity().Address, "received response")
	response, err := p.generateResponse(responses, secret, challenge.Challenge.CosiChallenge)
	if err != nil {
		return err
	}
	err = p.SendToParent(&Response{response})
	if err != nil {
		return err
	}

	// ----- Final Signature -----
	if p.IsRoot() {
		log.Lvl3(p.ServerIdentity().Address, "starts final signature")
		var signature []byte
		signature, err = cosi.Sign(p.Suite(), commitment, response, mask)
		if err != nil {
			return err
		}
		p.FinalSignature <- signature
		log.Lvl3("Root-node is done")
		return nil

	}

	return nil
}

// generateCommitment generates a personal secret and commitment
// and returns respectively the secret, an aggregated commitment and an aggregated mask
func (p *Cosi) generateCommitment(structCommitments []StructCommitment) (abstract.Scalar, abstract.Point, *cosi.Mask, error) {

	//extract lists of commitments and masks
	var commitments []abstract.Point
	var masks [][]byte
	for _, c := range structCommitments {
		commitments = append(commitments, c.CosiCommitment)
		masks = append(masks, c.Mask)
	}

	//generate personal secret and commitment
	secret, commitment := cosi.Commit(p.Suite(), nil)
	commitments = append(commitments, commitment)

	//generate personal mask
	personalMask, err := cosi.NewMask(p.Suite(), p.List, p.TreeNode().PublicAggregateSubTree)
	if err != nil {
		return nil, nil, nil, err
	}
	masks = append(masks, personalMask.Mask())

	//aggregate commitments and masks
	aggCommitment, aggMask, err :=
		cosi.AggregateCommitments(p.Suite(), commitments, masks)
	if err != nil {
		return nil, nil, nil, err
	}

	//create final aggregated mask
	finalMask, err := cosi.NewMask(p.Suite(), p.List, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	finalMask.SetMask(aggMask)

	log.Lvl3(p.ServerIdentity().Address, "is done aggregating commitments with total of",
		len(commitments), "commitments")

	return secret, aggCommitment, finalMask, nil
}

// generateResponse generates a personal response based on the secret
// and returns the aggregated response of all children and the node
func (p *Cosi) generateResponse(structResponse []StructResponse, secret abstract.Scalar, challenge abstract.Scalar) (abstract.Scalar, error) {

	//extract lists of responses
	var responses []abstract.Scalar
	for _, c := range structResponse {
		responses = append(responses, c.CosiReponse)
	}

	//generate personal response
	personalResponse, err := cosi.Response(p.Suite(), p.TreeNodeInstance.Private(), secret, challenge)
	if err != nil {
		return nil, err
	}
	responses = append(responses, personalResponse)

	//aggregate responses
	aggResponse, err := cosi.AggregateResponses(p.Suite(), responses)
	if err != nil {
		return nil, err
	}

	log.Lvl3(p.ServerIdentity().Address, "is done aggregating responses with total of",
		len(responses), "responses")

	return aggResponse, nil
}